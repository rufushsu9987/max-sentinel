variable "aws_region" {
  type        = string
  description = "AWS region."
  default     = "ap-northeast-1"
}

variable "environment" {
  type        = string
  description = "Deployment environment."
  default     = "dev"
}

variable "owner" {
  type        = string
  description = "Resource owner tag."
  default     = "rufushsu"
}

variable "kubernetes_version" {
  type        = string
  description = "Amazon EKS Kubernetes minor version."
  default     = "1.36"
}

variable "kafka_version" {
  type        = string
  description = "Amazon MSK Apache Kafka version."
  default     = "3.9.x"
}

variable "db_username" {
  type        = string
  description = "PostgreSQL master username."
  default     = "maxsentinel"
}

variable "db_password" {
  type        = string
  description = "PostgreSQL password. Supply through TF_VAR_db_password or a secret backend."
  sensitive   = true
}

variable "kafka_username" {
  type        = string
  description = "MSK SASL/SCRAM username."
  default     = "maxsentinel"
}

variable "kafka_password" {
  type        = string
  description = "MSK SASL/SCRAM password. Supply through TF_VAR_kafka_password or a secret backend."
  sensitive   = true
}
