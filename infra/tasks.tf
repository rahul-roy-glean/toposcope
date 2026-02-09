resource "google_cloud_tasks_queue" "ingestion" {
  name     = "${local.service_name}-ingestion"
  location = var.region

  rate_limits {
    max_dispatches_per_second = 5
    max_concurrent_dispatches = 3
  }

  retry_config {
    max_attempts       = 3
    min_backoff        = "10s"
    max_backoff        = "300s"
    max_doublings      = 3
    max_retry_duration = "3600s"
  }
}
