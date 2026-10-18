module Billing
  class Report
    def publish
      render
      upload
    end

    def render
      'report'
    end
  end
end
