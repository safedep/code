module Billing
  class Runner
    def go
      Report.new.emit
    end
  end
end
