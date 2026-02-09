resource "google_sql_database_instance" "main" {
  name             = "${local.service_name}-db-${local.env}"
  database_version = "POSTGRES_15"
  region           = var.region

  depends_on = [google_service_networking_connection.private_vpc]

  settings {
    tier              = var.db_tier
    availability_type = local.env == "prod" ? "REGIONAL" : "ZONAL"
    disk_autoresize   = true
    disk_size         = 10
    disk_type         = "PD_SSD"

    ip_configuration {
      ipv4_enabled                                  = false
      private_network                               = google_compute_network.main.id
      enable_private_path_for_google_cloud_services = true
    }

    backup_configuration {
      enabled                        = true
      point_in_time_recovery_enabled = local.env == "prod"
      start_time                     = "03:00"
    }

    database_flags {
      name  = "max_connections"
      value = "100"
    }

    user_labels = local.labels
  }

  deletion_protection = local.env == "prod"
}

resource "google_sql_database" "main" {
  name     = "toposcope"
  instance = google_sql_database_instance.main.name
}

resource "google_sql_user" "main" {
  name     = "toposcope"
  instance = google_sql_database_instance.main.name
  password = var.db_password
}
