import okpkg


def later():
    import badpkg

    badpkg.load()


okpkg.run()
