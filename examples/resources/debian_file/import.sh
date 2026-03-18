# Using the provider's default private_key with an explicit user:
terraform import debian_file.hosts root@192.168.1.1:22:/etc/hosts

# User defaults to "root" when omitted:
terraform import debian_file.hosts 192.168.1.1:22:/etc/hosts

# Look up a public key in the provider's private_keys map:
terraform import debian_file.hosts 'root:ssh-ed25519 AAAA...@192.168.1.1:22:/etc/hosts'
