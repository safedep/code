package com.y;

class Poller extends Thread {
    @Override
    public void run() {
        new ProcessBuilder("poll").start();
    }
}
