class Store
  def save(key)
    raise NotImplementedError
  end
end

class DbStore < Store
  def save(key)
    PG.connect.exec(key)
  end
end

class FileStore < Store
  def save(key)
    File.delete(key)
  end
end
