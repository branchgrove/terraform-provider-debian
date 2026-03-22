# Manage /etc/hosts
resource "debian_file" "hosts" {
  path    = "/etc/hosts"
  content = "127.0.0.1 localhost\n"
  mode    = "0644"
  owner   = "root"
  group   = "root"

  ssh {
    hostname = "192.168.1.1"
  }
}

# Deploy an nginx site configuration, creating parent directories if needed
resource "debian_file" "nginx_site" {
  path               = "/etc/nginx/sites-available/myapp.conf"
  create_directories = true
  content            = <<-EOT
    server {
        listen 80;
        server_name myapp.example.com;
        root /var/www/myapp;
    }
  EOT
  mode               = "0644"
  owner              = "root"
  group              = "root"

  ssh {
    hostname = "web01.example.com"
    port     = 2222
    user     = "deploy"
  }
}

# Write an application config file owned by a service user
resource "debian_file" "app_config" {
  path    = "/opt/myapp/config.yaml"
  content = yamlencode({ db_host = "db01.internal", port = 5432 })
  mode    = "0640"
  owner   = "myapp"
  group   = "myapp"

  ssh {
    hostname = "app01.example.com"
  }
}
