# AWS Terraform Reference Deployment

This directory demonstrates a production-oriented AWS baseline:

- private Amazon EKS control-plane endpoint
- managed node group in private subnets
- encrypted Amazon RDS for PostgreSQL
- Amazon MSK with TLS and SASL/SCRAM
- KMS keys and Secrets Manager for Kafka credentials
- private networking, backups, logs, and resource tags

## Cost and safety warning

This is a **reference architecture**, not a free-tier deployment. EKS, NAT Gateway, MSK brokers, RDS, data transfer, logs, and public load balancers can generate ongoing charges. Review the Terraform plan and AWS Pricing Calculator before applying. Use an isolated AWS account and destroy non-production resources after the demo.

## Prerequisites

- Terraform 1.8+
- AWS CLI credentials with permission to create the declared resources
- an S3 + DynamoDB remote-state backend for team or long-lived use
- Go services published to a private container registry

## Usage

```bash
cp terraform.tfvars.example terraform.tfvars
export TF_VAR_db_password='replace-with-a-strong-random-password'
export TF_VAR_kafka_password='replace-with-a-different-strong-random-password'
terraform init
terraform fmt -check
terraform validate
terraform plan
```

Do not commit `terraform.tfvars`, plan files, state files, passwords, broker strings, or database endpoints.

## Deployment handoff

1. Apply the infrastructure after review.
2. Read the sensitive outputs locally.
3. Store application secrets through External Secrets Operator or another secret manager integration rather than committing Kubernetes Secret values.
4. Replace the placeholder images in `deploy/k8s/all.yaml`.
5. Configure private cluster access through VPN, bastion, or a controlled CI runner.
6. Apply the Kubernetes manifests and verify NetworkPolicy behavior before exposing the Gateway.

## Production hardening not included

- AWS WAF/CloudFront or an authenticated API gateway
- multi-account separation and centralized audit logging
- GuardDuty, Security Hub, Config, Inspector, and image-signature enforcement
- OpenTelemetry collector and managed Prometheus/Grafana
- automated database credential rotation
- Terraform remote-state resources and organization policy
