class User < ApplicationRecord
  def profile
    Digest::SHA256.hexdigest(name)
  end

  def purge
    FileUtils.rm_rf(path)
  end
end
