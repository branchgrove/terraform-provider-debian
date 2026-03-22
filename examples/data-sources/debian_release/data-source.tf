# Read the Debian release information
data "debian_release" "this" {
  ssh {
    hostname = "app01.example.com"
  }
}

output "debian_version" {
  value = "${data.debian_release.this.pretty_name} (${data.debian_release.this.version_codename})"
}
