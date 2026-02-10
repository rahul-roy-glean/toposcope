# --- Global HTTP(S) Load Balancer with IAP for toposcope ---

# Reserve a static external IP
resource "google_compute_global_address" "lb" {
  name = "${local.service_name}-lb-ip"
}

# Serverless NEG pointing to the Cloud Run service
resource "google_compute_region_network_endpoint_group" "main" {
  name                  = local.service_name
  region                = var.region
  network_endpoint_type = "SERVERLESS"

  cloud_run {
    service = google_cloud_run_v2_service.main.name
  }
}

# Backend service with IAP enabled
resource "google_compute_backend_service" "main" {
  name                  = "${local.service_name}-backend"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  protocol              = "HTTPS"
  timeout_sec           = 300

  backend {
    group = google_compute_region_network_endpoint_group.main.id
  }

  iap {
    oauth2_client_id     = var.iap_oauth_client_id
    oauth2_client_secret = var.iap_oauth_client_secret
  }

  log_config {
    enable = false
  }
}

# URL map (all traffic to the backend)
resource "google_compute_url_map" "main" {
  name            = local.service_name
  default_service = google_compute_backend_service.main.id
}

# Google-managed SSL certificate
resource "google_compute_managed_ssl_certificate" "main" {
  name = local.service_name

  managed {
    domains = [var.domain]
  }
}

# HTTPS proxy
resource "google_compute_target_https_proxy" "main" {
  name             = "${local.service_name}-target-proxy"
  url_map          = google_compute_url_map.main.id
  ssl_certificates = [google_compute_managed_ssl_certificate.main.id]
}

# Forwarding rule (static IP → HTTPS proxy)
resource "google_compute_global_forwarding_rule" "main" {
  name                  = local.service_name
  ip_address            = google_compute_global_address.lb.id
  ip_protocol           = "TCP"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  port_range            = "443"
  target                = google_compute_target_https_proxy.main.id
}

# Grant all Glean users access through IAP
resource "google_iap_web_backend_service_iam_member" "glean_users" {
  web_backend_service = google_compute_backend_service.main.name
  role                = "roles/iap.httpsResourceAccessor"
  member              = "domain:glean.com"
}
