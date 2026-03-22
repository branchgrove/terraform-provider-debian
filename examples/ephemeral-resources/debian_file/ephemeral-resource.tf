# Read a TLS private key without persisting to state
ephemeral "debian_file" "tls_key" {
  path = "/etc/ssl/private/server.key"

  ssh {
    hostname = "web01.example.com"
  }
}

# Read a secret file with a size limit
ephemeral "debian_file" "api_token" {
  path     = "/run/secrets/api_token"
  max_size = 1024

  ssh {
    hostname = "app01.example.com"
  }
}
