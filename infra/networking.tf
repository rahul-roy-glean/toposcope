resource "google_compute_network" "main" {
  name                    = "${local.service_name}-vpc"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "main" {
  name          = "${local.service_name}-subnet"
  ip_cidr_range = "10.0.0.0/20"
  region        = var.region
  network       = google_compute_network.main.id

  private_ip_google_access = true
}

# Private services access for Cloud SQL
resource "google_compute_global_address" "private_ip" {
  name          = "${local.service_name}-private-ip"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 16
  network       = google_compute_network.main.id
}

resource "google_service_networking_connection" "private_vpc" {
  network                 = google_compute_network.main.id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_ip.name]
}

# VPC connector for Cloud Run to access private resources
resource "google_vpc_access_connector" "main" {
  name          = "${local.service_name}-connector"
  region        = var.region
  network       = google_compute_network.main.name
  ip_cidr_range = "10.8.0.0/28"

  min_instances = 2
  max_instances = 3
}
