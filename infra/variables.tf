variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region"
  type        = string
  default     = "us-central1"
}

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "dev"
}

variable "db_tier" {
  description = "Cloud SQL machine tier. Use db-f1-micro for dev, db-custom-2-7680 or higher for production."
  type        = string
  default     = "db-f1-micro"
}

variable "db_password" {
  description = "Database password for the toposcope user"
  type        = string
  sensitive   = true
}

variable "github_app_id" {
  description = "GitHub App ID"
  type        = string
}

variable "github_webhook_secret" {
  description = "GitHub webhook secret for HMAC verification"
  type        = string
  sensitive   = true
}

variable "container_image" {
  description = "Container image for the Cloud Run service"
  type        = string
  default     = "gcr.io/toposcope/toposcoped:latest"
}

variable "extraction_worker_image" {
  description = "Container image for the extraction worker"
  type        = string
  default     = "gcr.io/toposcope/extraction-worker:latest"
}
