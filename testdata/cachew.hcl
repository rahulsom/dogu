state = "/tmp/cachew"
bind  = "0.0.0.0:8080"
url   = "http://localhost:8080/"

log {
  level = "info"
}

opa {
  policy = <<EOF
    package cachew.authz
    default allow := true
  EOF
}

metadata "memory" {}

cache "memory" {
  limit-mb = 512
}

strategy "host" "https://archive.apache.org" {}
strategy "host" "https://nodejs.org" {}
strategy "host" "https://services.gradle.org" {}
strategy "host" "https://registry.npmjs.org" {}
strategy "host" "https://github.com" {}
strategy "host" "https://objects.githubusercontent.com" {}
strategy "host" "https://raw.githubusercontent.com" {}
strategy "host" "https://api.github.com" {}
strategy "host" "https://pypi.org" {}
strategy "host" "https://api.foojay.io" {}
strategy "host" "https://cdn.azul.com" {}
