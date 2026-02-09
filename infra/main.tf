terraform {
  required_version = ">= 1.5"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
  }

  backend "gcs" {
    bucket = "toposcope-terraform-state"
    prefix = "terraform/state"
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

locals {
  service_name = "toposcope"
  env          = var.environment
  labels = {
    app         = local.service_name
    environment = local.env
    managed_by  = "terraform"
  }
}
