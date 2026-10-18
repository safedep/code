package indexer

import (
	"strings"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	"github.com/stretchr/testify/assert"
)

func TestFileIndexRuby(t *testing.T) {
	source := strings.Join([]string{
		"require_relative 'lib/store'",
		"require name",
		"class Job < Base",
		"  include Retry",
		"  before_action :check",
		"  attr_reader :name",
		"  alias_method :start, :run",
		"  def self.build; new; end",
		"  def run; send(:check); method(:check); eval('1'); end",
		"  def check; end",
		"  Point = Struct.new(:x) do",
		"    def show; end",
		"  end",
		"end",
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodeRuby, "job.rb", source)

	t.Run("a class has its superclass and its mixins", func(t *testing.T) {
		assert.Equal(t, []string{"Base", "Retry"}, classOf(t, ix, "job.rb//Job").Bases)
	})
	t.Run("a class method has its own key and the name that a call writes", func(t *testing.T) {
		build := definitionOf(t, ix, "job.rb//Job//self.build")
		assert.True(t, build.Static)
		assert.Equal(t, "build", build.Name)
		assert.False(t, definitionOf(t, ix, "job.rb//Job//run").Static)
	})
	t.Run("a require_relative is an import and a computed require is a flag", func(t *testing.T) {
		var modules []string
		for _, imp := range ix.Imports {
			modules = append(modules, string(imp.Kind)+" "+imp.Module)
		}
		assert.Equal(t, []string{"static ./lib/store", "dynamic-computed "}, modules)
		assert.Contains(t, ix.Flags, fileindex.Flag{Kind: fileindex.FlagComputedImport, Line: 2})
		assert.Contains(t, ix.Flags, fileindex.Flag{Kind: fileindex.FlagEval, Line: 9})
		assert.Contains(t, ix.Flags, fileindex.Flag{Kind: fileindex.FlagMonkeyPatch, Line: 11})
	})
	t.Run("a symbol that names a method is a reference", func(t *testing.T) {
		var targets []string
		for _, ref := range ix.References {
			targets = append(targets, ref.Targets...)
		}
		assert.Equal(t, []string{"job.rb//Job//check", "job.rb//Job//run", "job.rb//Job//check"}, targets)
	})
	t.Run("a send of a symbol is a call on an unknown receiver", func(t *testing.T) {
		var methods []string
		for _, u := range ix.Unresolved {
			if u.Reason == fileindex.UnresolvedUnknownReceiver {
				methods = append(methods, u.Method)
			}
		}
		assert.Contains(t, methods, "check")
	})
}
