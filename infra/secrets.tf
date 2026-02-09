resource "google_secret_manager_secret" "github_private_key" {
  secret_id = "${local.service_name}-github-private-key"

  replication {
    auto {}
  }

  labels = local.labels
}

resource "google_secret_manager_secret" "github_webhook_secret" {
  secret_id = "${local.service_name}-github-webhook-secret"

  replication {
    auto {}
  }

  labels = local.labels
}

# Store webhook secret value
resource "google_secret_manager_secret_version" "github_webhook_secret" {
  secret      = google_secret_manager_secret.github_webhook_secret.id
  secret_data = var.github_webhook_secret
}
