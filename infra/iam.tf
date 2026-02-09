# Service account for the Cloud Run service
resource "google_service_account" "service" {
  account_id   = "${local.service_name}-service"
  display_name = "Toposcope Service"
  description  = "Service account for the toposcoped Cloud Run service"
}

# Service account for extraction workers
resource "google_service_account" "extraction" {
  account_id   = "${local.service_name}-extraction"
  display_name = "Toposcope Extraction Worker"
  description  = "Service account for Cloud Batch extraction jobs"
}

# Service account can access Cloud SQL
resource "google_project_iam_member" "service_cloudsql" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.service.email}"
}

# Service account can read/write GCS bucket
resource "google_storage_bucket_iam_member" "service_storage" {
  bucket = google_storage_bucket.data.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.service.email}"
}

# Service account can access secrets
resource "google_secret_manager_secret_iam_member" "service_github_key" {
  secret_id = google_secret_manager_secret.github_private_key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.service.email}"
}

resource "google_secret_manager_secret_iam_member" "service_webhook_secret" {
  secret_id = google_secret_manager_secret.github_webhook_secret.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.service.email}"
}

# Service account can enqueue Cloud Tasks
resource "google_project_iam_member" "service_tasks" {
  project = var.project_id
  role    = "roles/cloudtasks.enqueuer"
  member  = "serviceAccount:${google_service_account.service.email}"
}

# Service account can submit Cloud Batch jobs
resource "google_project_iam_member" "service_batch" {
  project = var.project_id
  role    = "roles/batch.jobsEditor"
  member  = "serviceAccount:${google_service_account.service.email}"
}

# Extraction worker can read/write GCS bucket
resource "google_storage_bucket_iam_member" "extraction_storage" {
  bucket = google_storage_bucket.data.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.extraction.email}"
}

# Extraction worker can pull container images
resource "google_project_iam_member" "extraction_artifacts" {
  project = var.project_id
  role    = "roles/artifactregistry.reader"
  member  = "serviceAccount:${google_service_account.extraction.email}"
}
