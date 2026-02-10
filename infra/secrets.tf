# No secrets needed for CI-based ingest flow.
# API key is passed as a plain env var on Cloud Run.
# If you need to store the API key in Secret Manager instead,
# create a secret here and reference it via value_source in compute.tf.
