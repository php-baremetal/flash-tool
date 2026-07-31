<?php
// setup()/loop() sketch: blink an LED on GPIO2.
function setup() {
    gpio_mode(2, GPIO_OUTPUT);
}
function loop($tick) {
    gpio_write(2, $tick % 2);
    delay(500);
}
