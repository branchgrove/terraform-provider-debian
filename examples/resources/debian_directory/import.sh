# Using the provider's default private_key:
terraform import debian_directory.app_data 'user=root;host=app01.example.com;port=22;id=/opt/myapp/data'

# User defaults to "root" when omitted:
terraform import debian_directory.app_data 'host=app01.example.com;port=22;id=/opt/myapp/data'

# Look up a public key in the provider's private_keys map:
terraform import debian_directory.app_data 'user=root;host=app01.example.com;port=22;public_key=ssh-ed25519 AAAA...;id=/opt/myapp/data'
