# Add a user to the sudo group
resource "debian_group_member" "alice_sudo" {
  user  = "alice"
  group = "sudo"

  ssh {
    hostname = "dev01.example.com"
  }
}

# Add a deploy user to the www-data group
resource "debian_group_member" "deploy_www" {
  user  = "deploy"
  group = "www-data"

  ssh {
    hostname = "web01.example.com"
  }
}
