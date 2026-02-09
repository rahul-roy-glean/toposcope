output "service_url" {
  description = "Cloud Run service URL"
  value       = google_cloud_run_v2_service.main.uri
}

output "database_connection" {
  description = "Cloud SQL connection name"
  value       = google_sql_database_instance.main.connection_name
}

output "database_private_ip" {
  description = "Cloud SQL private IP address"
  value       = google_sql_database_instance.main.private_ip_address
  sensitive   = true
}

output "storage_bucket" {
  description = "GCS bucket name"
  value       = google_storage_bucket.data.name
}

output "service_account_email" {
  description = "Service account email for the Cloud Run service"
  value       = google_service_account.service.email
}

output "extraction_service_account_email" {
  description = "Service account email for extraction workers"
  value       = google_service_account.extraction.email
}

output "cloud_tasks_queue" {
  description = "Cloud Tasks queue ID"
  value       = google_cloud_tasks_queue.ingestion.id
}

output "vpc_connector" {
  description = "VPC connector name"
  value       = google_vpc_access_connector.main.name
}
