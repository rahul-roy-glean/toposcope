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

variable "api_key" {
  description = "API key for authenticating ingest requests. Leave empty to disable auth."
  type        = string
  default     = ""
  sensitive   = true
}

variable "container_image" {
  description = "Container image for the API Cloud Run service"
  type        = string
  default     = "us-central1-docker.pkg.dev/scio-ci/glean-images/toposcoped:v0.3.0"
}

variable "web_container_image" {
  description = "Container image for the web UI Cloud Run service"
  type        = string
  default     = "us-central1-docker.pkg.dev/scio-ci/glean-images/toposcope-web:v0.3.0"
}

variable "domain" {
  description = "Domain for the toposcope service (e.g., toposcope.internal.glean.com)"
  type        = string
  default     = "toposcope.internal.glean.com"
}

variable "iap_oauth_client_id" {
  description = "OAuth client ID for IAP"
  type        = string
}

variable "iap_oauth_client_secret" {
  description = "OAuth client secret for IAP"
  type        = string
  sensitive   = true
}

variable "github_repo" {
  description = "GitHub repo allowed to authenticate via WIF (e.g. 'glean/mono')"
  type        = string
  default     = "askscio/scio"
}
