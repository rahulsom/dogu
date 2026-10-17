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
