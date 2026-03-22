# Read a secret from a vault without persisting to state
ephemeral "debian_command" "db_password" {
  command = "cat /run/secrets/db_password"

  ssh {
    hostname = "app01.example.com"
  }
}

# Generate a temporary token
ephemeral "debian_command" "temp_token" {
  command = "openssl rand -hex 32"

  ssh {
    hostname = "app01.example.com"
  }
}

# Run a command with stdin and environment variables
ephemeral "debian_command" "decrypt" {
  command = "gpg --batch --decrypt --passphrase-fd 0"
  stdin   = var.gpg_passphrase
  env = {
    GNUPGHOME = "/opt/myapp/.gnupg"
  }

  ssh {
    hostname = "app01.example.com"
  }
}
