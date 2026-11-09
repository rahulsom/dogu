state = "/var/cache/cachew"
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

// Persist both objects and their TTL database in the mounted state directory.
cache "disk" {
  limit-mb = 4096
  max-ttl = "168h"
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
strategy "host" "https://release-assets.githubusercontent.com" {}
strategy "host" "https://downloads.gradle.org" {}
strategy "host" "https://dlcdn.apache.org" {}

// Hosts exercised by the generated documentation examples.
strategy "host" "https://download.clojure.org" {}
strategy "host" "https://go.dev" {}
strategy "host" "https://dl.google.com" {}
strategy "host" "https://static.rust-lang.org" {}
strategy "host" "https://downloads.haskell.org" {}
strategy "host" "https://cran.r-project.org" {}
strategy "host" "https://julialang-s3.julialang.org" {}
strategy "host" "https://cache.ruby-lang.org" {}
strategy "host" "https://www.lua.org" {}
strategy "host" "https://rubygems.org" {}
strategy "host" "https://index.rubygems.org" {}
strategy "host" "https://proxy.golang.org" {}
strategy "host" "https://storage.googleapis.com" {}
strategy "host" "https://gitlab.com" {}
strategy "host" "https://graphviz.org" {}
strategy "host" "https://hackage.haskell.org" {}
strategy "host" "https://sourceforge.net" {}
strategy "host" "https://downloads.sourceforge.net" {}
strategy "host" "https://master.dl.sourceforge.net" {}
