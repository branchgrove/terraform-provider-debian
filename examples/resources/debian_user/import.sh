# Using the provider's default private_key:
terraform import debian_user.myapp 'user=root;host=app01.example.com;port=22;id=myapp'

# User defaults to "root" when omitted:
terraform import debian_user.myapp 'host=app01.example.com;port=22;id=myapp'

# Look up a public key in the provider's private_keys map:
terraform import debian_user.myapp 'user=root;host=app01.example.com;port=22;public_key=ssh-ed25519 AAAA...;id=myapp'
