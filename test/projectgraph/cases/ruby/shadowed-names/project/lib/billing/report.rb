module Billing
  class Report
    def emit
      Kernel.exec('billing report')
    end
  end
end
