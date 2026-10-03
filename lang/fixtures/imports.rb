require 'openai'
require "net/http"
require_relative 'lib/helper'
require "active_support/core_ext/#{name}"

begin
  require 'google/cloud/storage'
rescue LoadError
end

class Agent
  def run(prompt)
    OpenAI::Client.new.chat(parameters: { model: "gpt-4o" })
  end

  def self.build
    new
  end
end
