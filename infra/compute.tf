resource "google_cloud_run_v2_service" "main" {
  name     = "${local.service_name}-service"
  location = var.region

  template {
    service_account = google_service_account.service.email

    scaling {
      min_instance_count = local.env == "prod" ? 1 : 0
      max_instance_count = local.env == "prod" ? 10 : 2
    }

    vpc_access {
      connector = google_vpc_access_connector.main.id
      egress    = "PRIVATE_RANGES_ONLY"
    }

    containers {
      image = var.container_image

      ports {
        container_port = 8080
      }

      env {
        name  = "PORT"
        value = "8080"
      }
      env {
        name  = "ENVIRONMENT"
        value = local.env
      }
      env {
        name  = "PROJECT_ID"
        value = var.project_id
      }
      env {
        name  = "DATABASE_URL"
        value = "postgres://toposcope:${var.db_password}@${google_sql_database_instance.main.private_ip_address}:5432/toposcope?sslmode=disable"
      }
      env {
        name  = "GCS_BUCKET"
        value = google_storage_bucket.data.name
      }
      env {
        name  = "GITHUB_APP_ID"
        value = var.github_app_id
      }
      env {
        name = "GITHUB_PRIVATE_KEY"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.github_private_key.id
            version = "latest"
          }
        }
      }
      env {
        name = "GITHUB_WEBHOOK_SECRET"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.github_webhook_secret.id
            version = "latest"
          }
        }
      }
      env {
        name  = "CLOUD_TASKS_QUEUE"
        value = google_cloud_tasks_queue.ingestion.id
      }
      env {
        name  = "EXTRACTION_WORKER_IMAGE"
        value = var.extraction_worker_image
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
      }

      startup_probe {
        http_get {
          path = "/healthz"
        }
        initial_delay_seconds = 5
        period_seconds        = 3
      }

      liveness_probe {
        http_get {
          path = "/healthz"
        }
        period_seconds = 30
      }
    }
  }

  labels = local.labels
}

# Allow unauthenticated access (GitHub webhooks)
resource "google_cloud_run_v2_service_iam_member" "public" {
  name     = google_cloud_run_v2_service.main.name
  location = google_cloud_run_v2_service.main.location
  role     = "roles/run.invoker"
  member   = "allUsers"
}
