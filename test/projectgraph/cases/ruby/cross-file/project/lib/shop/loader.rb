require 'yaml'

module Shop
  class Loader
    def initialize(path)
      @path = path
    end

    def load
      YAML.load_file(@path)
    end

    def dump(data)
      YAML.dump(data)
    end
  end
end
