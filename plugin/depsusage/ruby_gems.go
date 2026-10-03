package depsusage

// rubyRequireGems maps a require path to the gem that provides it, when the
// first segment of the path is not the gem name. A key matches the path and
// the paths below it.
var rubyRequireGems = map[string]string{
	"action_cable":      "actioncable",
	"action_controller": "actionpack",
	"action_dispatch":   "actionpack",
	"action_mailer":     "actionmailer",
	"action_view":       "actionview",
	"active_job":        "activejob",
	"active_model":      "activemodel",
	"active_record":     "activerecord",
	"active_storage":    "activestorage",
	"active_support":    "activesupport",
	"langchain":         "langchainrb",
	"openai":            "ruby-openai",
}

// rubyHyphenatedRoots are require path roots of gem families, such as
// google/cloud/storage from google-cloud-storage. The value is the number of
// path segments in the gem name.
var rubyHyphenatedRoots = map[string]int{
	"google/apis":  3,
	"google/cloud": 3,
}
