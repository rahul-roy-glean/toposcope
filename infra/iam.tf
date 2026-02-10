# Service account for the Cloud Run service
resource "google_service_account" "service" {
  account_id   = "${local.service_name}-service"
  display_name = "Toposcope Service"
  description  = "Service account for the toposcoped Cloud Run service"
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

# --- CI service account (used by GitHub Actions via WIF) ---

resource "google_service_account" "ci" {
  account_id   = "${local.service_name}-ci"
  display_name = "Toposcope CI"
  description  = "Service account for GitHub Actions to call toposcoped"
}

# CI SA can invoke the Cloud Run service
resource "google_cloud_run_v2_service_iam_member" "ci_invoker" {
  name     = google_cloud_run_v2_service.main.name
  location = google_cloud_run_v2_service.main.location
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.ci.email}"
}

# --- Workload Identity Federation ---

resource "google_iam_workload_identity_pool" "github" {
  workload_identity_pool_id = "${local.service_name}-github"
  display_name              = "Toposcope GitHub Actions"
  description               = "WIF pool for GitHub Actions"
}

resource "google_iam_workload_identity_pool_provider" "github" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = "github-actions"
  display_name                       = "GitHub Actions"

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.actor"      = "assertion.actor"
    "attribute.repository" = "assertion.repository"
  }

  attribute_condition = "assertion.repository == \"${var.github_repo}\""

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

# Allow the GitHub repo to impersonate the CI service account
resource "google_service_account_iam_member" "ci_wif" {
  service_account_id = google_service_account.ci.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.repository/${var.github_repo}"
}
