package highmc

const (
	// Version is a version of this server.
	Version = "1.1.0 alpha-dev"
)

// GitCommit is a git commit hash for this project.
// You should set this with -ldflags "-X github.com/cr0sh/highmc.GitVersion="
var GitCommit = "unknown"

// BuildTime is a timestamp when the program is built.
// You should set this with -ldflags "-X github.com/cr0sh/highmc.BuildTime="
var BuildTime = "unknown"

var lastEntityID = uint64(1)

var levels = map[string]*Level{
	defaultLvl: {Name: "dummy"},
}

var defaultLvl = "default"
