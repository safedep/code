require 'shop/loader'

module Shop
  class Runner
    def run(path)
      data = Loader.new(path).load
      report(data)
    end

    def report(data)
      puts data
    end
  end
end
