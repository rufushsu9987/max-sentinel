output "cluster_name" {
  value = module.eks.cluster_name
}

output "configure_kubectl" {
  value = "aws eks update-kubeconfig --region ${var.aws_region} --name ${module.eks.cluster_name}"
}

output "postgres_endpoint" {
  value     = aws_db_instance.postgres.address
  sensitive = true
}

output "msk_bootstrap_brokers_sasl_scram" {
  value     = aws_msk_cluster.events.bootstrap_brokers_sasl_scram
  sensitive = true
}

output "msk_scram_secret_arn" {
  value = aws_secretsmanager_secret.msk_scram.arn
}
