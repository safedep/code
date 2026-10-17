require 'yaml'
require_relative 'util/helper'

module Shop
  class App < Base
    include Audits
    before_action :audit

    def initialize(store)
      @store = store
    end

    def run(path)
      @store.save(YAML.load_file(path))
      audit
      Helper.assist(path)
      send(:audit)
    end

    def self.main(argv)
      new(Store.new).run(argv[0])
    end

    private

    def audit
      puts 'ran'
    end
  end
end
