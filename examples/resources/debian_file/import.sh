# Using the provider's default private_key:
terraform import debian_file.hosts 'user=root;host=192.168.1.1;port=22;id=/etc/hosts'

# User defaults to "root" when omitted:
terraform import debian_file.hosts 'host=192.168.1.1;port=22;id=/etc/hosts'

# Look up a public key in the provider's private_keys map:
terraform import debian_file.hosts 'user=root;host=192.168.1.1;port=22;public_key=ssh-ed25519 AAAA...;id=/etc/hosts'
