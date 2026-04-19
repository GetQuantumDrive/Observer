plugins {
    java
    id("io.getquantumdrive.observer")
}

observer {
    failOn = "never" // don't break fixture build; tests inspect the report
}
