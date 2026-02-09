resource "google_storage_bucket" "data" {
  name          = "${var.project_id}-toposcope-data"
  location      = var.region
  force_destroy = local.env != "prod"

  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"

  versioning {
    enabled = local.env == "prod"
  }

  lifecycle_rule {
    condition {
      age = 90
    }
    action {
      type          = "SetStorageClass"
      storage_class = "NEARLINE"
    }
  }

  lifecycle_rule {
    condition {
      age = 365
    }
    action {
      type          = "SetStorageClass"
      storage_class = "COLDLINE"
    }
  }

  labels = local.labels
}
