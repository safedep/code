require 'net/http'

module Billing
  class Report
    def upload
      Net::HTTP.post(URI('https://example.com'), render)
    end
  end
end
