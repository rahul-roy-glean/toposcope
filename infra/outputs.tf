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

output "vpc_connector" {
  description = "VPC connector name"
  value       = google_vpc_access_connector.main.name
}

output "ci_service_account_email" {
  description = "CI service account email (for GitHub Actions)"
  value       = google_service_account.ci.email
}

output "lb_ip_address" {
  description = "Static IP address for the load balancer. Create a DNS A record pointing your domain here."
  value       = google_compute_global_address.lb.address
}

output "workload_identity_provider" {
  description = "WIF provider resource name (for GitHub Actions auth)"
  value       = google_iam_workload_identity_pool_provider.github.name
}
