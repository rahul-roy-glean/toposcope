resource "google_cloud_run_v2_service" "main" {
  name     = "${local.service_name}-service"
  location = var.region

  ingress = "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER"

  template {
    service_account = google_service_account.service.email
    timeout         = "300s"

    scaling {
      min_instance_count = local.env == "prod" ? 1 : 0
      max_instance_count = local.env == "prod" ? 10 : 2
    }

    vpc_access {
      connector = google_vpc_access_connector.main.id
      egress    = "PRIVATE_RANGES_ONLY"
    }

    containers {
      image   = var.container_image
      command = ["toposcoped"]

      ports {
        container_port = 8080
      }

      env {
        name  = "ENVIRONMENT"
        value = local.env
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
        name  = "API_KEY"
        value = var.api_key
      }
      env {
        name  = "SNAPSHOT_CACHE_SIZE"
        value = "20"
      }
      env {
        name  = "LOCAL_STORAGE_PATH"
        value = "/tmp/toposcope-data"
      }

      resources {
        limits = {
          cpu    = "2"
          memory = "2Gi"
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
