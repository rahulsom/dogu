package tools

// The catalog describes release naming and installation metadata. Providers with
// custom release discovery (Java, Node, Python, Pants) keep their own code.
var (
	antDefinition = toolDefinition{
		name: "ant", baseURL: "https://archive.apache.org/dist/ant/binaries",
		source: versionSource{kind: apacheAntSource},
		asset:  "apache-ant-{version}-bin.tar.gz", downloadPath: "{asset}",
		homeEnv: []string{"ANT_HOME"},
	}
	mavenDefinition = toolDefinition{
		name: "maven", baseURL: "https://archive.apache.org/dist/maven/maven-3",
		source: versionSource{kind: apacheMavenSource},
		asset:  "apache-maven-{version}-bin.tar.gz", downloadPath: "{version}/binaries/{asset}",
		homeEnv: []string{"M2_HOME", "MAVEN_HOME"},
	}
	gradleDefinition = toolDefinition{
		name: "gradle", baseURL: "https://services.gradle.org/distributions",
		source: versionSource{kind: gradleSource},
		asset:  "gradle-{version}-bin.zip", downloadPath: "{asset}",
		homeEnv: []string{"GRADLE_HOME"},
	}
	sbtDefinition = toolDefinition{
		name: "sbt", baseURL: "https://github.com/sbt/sbt/releases/download",
		source: versionSource{kind: githubSource, owner: "sbt", repository: "sbt"},
		asset:  "sbt-{version}.{ext}", downloadPath: "v{version}/{asset}",
		extension: "tgz", windowsExtension: "zip", homeEnv: []string{"SBT_HOME"},
	}
	yarnDefinition = toolDefinition{
		name: "yarn", baseURL: "https://github.com/yarnpkg/yarn/releases/download",
		source: versionSource{kind: npmSource, packageName: "yarn"},
		asset:  "yarn-v{version}.tar.gz", downloadPath: "v{version}/{asset}",
	}
	bazelDefinition = toolDefinition{
		name: "bazel", baseURL: "https://github.com/bazelbuild/bazel/releases/download",
		source: versionSource{kind: githubSource, owner: "bazelbuild", repository: "bazel"},
		asset:  "bazel-{version}-{os}-{arch}{suffix}", downloadPath: "{version}/{asset}",
		binary: "bazel", windowsSuffix: ".exe", homeEnv: []string{"BAZEL_HOME"},
		platform: platformMapping{arch: "x86_64", arm64: "arm64", linux: "linux", darwin: "darwin", windows: "windows"},
	}
	buck2Definition = toolDefinition{
		name: "buck2", baseURL: "https://github.com/facebook/buck2/releases/download",
		source: versionSource{kind: githubSource, owner: "facebook", repository: "buck2"},
		asset:  "buck2-{arch}-{os}{suffix}.zst", downloadPath: "{version}/{asset}",
		binary: "buck2", windowsSuffix: ".exe", homeEnv: []string{"BUCK2_HOME"},
		platform: platformMapping{arch: "x86_64", arm64: "aarch64", linux: "unknown-linux-gnu", darwin: "apple-darwin", windows: "pc-windows-msvc"},
	}
	millDefinition = toolDefinition{
		name: "mill", baseURL: "https://github.com/com-lihaoyi/mill/releases/download",
		source: versionSource{kind: githubSource, owner: "com-lihaoyi", repository: "mill"},
		asset:  "{version}{suffix}", downloadPath: "{version}/{asset}",
		cacheName: "mill-{version}-{binary}", binary: "mill", windowsSuffix: ".bat",
		homeEnv: []string{"MILL_HOME"}, extraEnv: map[string]string{"MILL_VERSION": "{version}"},
	}
	groovyDefinition = toolDefinition{
		name: "groovy", baseURL: "https://archive.apache.org/dist/groovy",
		source: versionSource{kind: indexSource},
		asset:  "apache-groovy-binary-{version}.zip", downloadPath: "{version}/distribution/{asset}",
		homeEnv: []string{"GROOVY_HOME"},
	}
	kotlinDefinition = toolDefinition{
		name: "kotlin", baseURL: "https://github.com/JetBrains/kotlin/releases/download",
		source: versionSource{kind: githubSource, owner: "JetBrains", repository: "kotlin"},
		asset:  "kotlin-compiler-{version}.zip", downloadPath: "v{version}/{asset}",
		homeEnv: []string{"KOTLIN_HOME"},
	}
	scalaDefinition = toolDefinition{
		name: "scala", baseURL: "https://github.com/scala/scala/releases/download",
		source: versionSource{kind: githubSource, owner: "scala", repository: "scala"},
		asset:  "scala-{version}.zip", downloadPath: "v{version}/{asset}",
		homeEnv: []string{"SCALA_HOME"},
	}
	elmDefinition = toolDefinition{
		name: "elm", baseURL: "https://github.com/elm/compiler/releases/download",
		source: versionSource{kind: githubSource, owner: "elm", repository: "compiler"},
		asset:  "binary-for-{os}-64-bit.gz", downloadPath: "{version}/{asset}",
		binary: "elm", windowsSuffix: ".exe",
		platform: platformMapping{linux: "linux", darwin: "mac", windows: "windows"},
	}
)
