require_relative 'lib/stores'

def persist(store, key)
  store.save(key)
end

if __FILE__ == $PROGRAM_NAME
  persist(DbStore.new, ARGV[0])
end
