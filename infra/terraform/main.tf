data "aws_availability_zones" "available" {
  state = "available"
}

locals {
  name = "max-sentinel-${var.environment}"
  azs  = slice(data.aws_availability_zones.available.names, 0, 3)
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 6.0"

  name = local.name
  cidr = "10.42.0.0/16"
  azs  = local.azs

  private_subnets  = ["10.42.0.0/20", "10.42.16.0/20", "10.42.32.0/20"]
  public_subnets   = ["10.42.128.0/24", "10.42.129.0/24", "10.42.130.0/24"]
  database_subnets = ["10.42.160.0/24", "10.42.161.0/24", "10.42.162.0/24"]

  enable_nat_gateway   = true
  single_nat_gateway   = var.environment != "prod"
  enable_dns_hostnames = true

  public_subnet_tags = {
    "kubernetes.io/role/elb" = 1
  }
  private_subnet_tags = {
    "kubernetes.io/role/internal-elb" = 1
  }
}

module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 21.0"

  name               = local.name
  kubernetes_version = var.kubernetes_version
  vpc_id             = module.vpc.vpc_id
  subnet_ids         = module.vpc.private_subnets

  endpoint_private_access = true
  endpoint_public_access  = false
  enable_cluster_creator_admin_permissions = true

  addons = {
    coredns                = {}
    eks-pod-identity-agent = {}
    kube-proxy             = {}
    vpc-cni = {
      before_compute = true
    }
  }

  eks_managed_node_groups = {
    general = {
      instance_types = ["m7i.large"]
      min_size       = 2
      desired_size   = 2
      max_size       = 6
      capacity_type  = "ON_DEMAND"
    }
  }
}

resource "aws_security_group" "data" {
  name        = "${local.name}-data"
  description = "Data-plane access from the application VPC"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description = "PostgreSQL from VPC"
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    cidr_blocks = [module.vpc.vpc_cidr_block]
  }

  ingress {
    description = "MSK TLS with SASL/SCRAM from VPC"
    from_port   = 9096
    to_port     = 9096
    protocol    = "tcp"
    cidr_blocks = [module.vpc.vpc_cidr_block]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_db_instance" "postgres" {
  identifier                      = local.name
  engine                          = "postgres"
  engine_version                  = "16"
  instance_class                  = var.environment == "prod" ? "db.m7g.large" : "db.t4g.micro"
  allocated_storage               = 20
  max_allocated_storage           = 100
  storage_encrypted               = true
  db_name                         = "maxsentinel"
  username                        = var.db_username
  password                        = var.db_password
  db_subnet_group_name            = module.vpc.database_subnet_group_name
  vpc_security_group_ids          = [aws_security_group.data.id]
  multi_az                        = var.environment == "prod"
  backup_retention_period         = var.environment == "prod" ? 14 : 1
  deletion_protection             = var.environment == "prod"
  skip_final_snapshot             = var.environment != "prod"
  publicly_accessible             = false
  performance_insights_enabled    = var.environment == "prod"
  enabled_cloudwatch_logs_exports = ["postgresql", "upgrade"]
}

resource "aws_kms_key" "msk" {
  description             = "KMS key for ${local.name} MSK data and SCRAM secret"
  deletion_window_in_days = 14
  enable_key_rotation     = true
}

resource "aws_kms_alias" "msk" {
  name          = "alias/${local.name}-msk"
  target_key_id = aws_kms_key.msk.key_id
}

resource "aws_secretsmanager_secret" "msk_scram" {
  name       = "AmazonMSK_${replace(local.name, "-", "_")}"
  kms_key_id = aws_kms_key.msk.arn
}

resource "aws_secretsmanager_secret_version" "msk_scram" {
  secret_id = aws_secretsmanager_secret.msk_scram.id
  secret_string = jsonencode({
    username = var.kafka_username
    password = var.kafka_password
  })
}

resource "aws_msk_configuration" "events" {
  kafka_versions = [var.kafka_version]
  name           = "${local.name}-configuration"

  server_properties = <<-PROPERTIES
    auto.create.topics.enable=true
    default.replication.factor=2
    min.insync.replicas=1
    num.partitions=6
    unclean.leader.election.enable=false
  PROPERTIES
}

resource "aws_msk_cluster" "events" {
  cluster_name           = "${local.name}-events"
  kafka_version          = var.kafka_version
  number_of_broker_nodes = 2
  enhanced_monitoring    = "PER_BROKER"

  broker_node_group_info {
    client_subnets  = slice(module.vpc.private_subnets, 0, 2)
    instance_type   = "kafka.t3.small"
    security_groups = [aws_security_group.data.id]

    storage_info {
      ebs_storage_info {
        volume_size = 100
      }
    }
  }

  client_authentication {
    sasl {
      scram = true
    }
  }

  encryption_info {
    encryption_at_rest_kms_key_arn = aws_kms_key.msk.arn
    encryption_in_transit {
      client_broker = "TLS"
      in_cluster    = true
    }
  }

  configuration_info {
    arn      = aws_msk_configuration.events.arn
    revision = aws_msk_configuration.events.latest_revision
  }

  open_monitoring {
    prometheus {
      jmx_exporter {
        enabled_in_broker = true
      }
      node_exporter {
        enabled_in_broker = true
      }
    }
  }
}

resource "aws_msk_scram_secret_association" "events" {
  cluster_arn     = aws_msk_cluster.events.arn
  secret_arn_list = [aws_secretsmanager_secret.msk_scram.arn]

  depends_on = [aws_secretsmanager_secret_version.msk_scram]
}
