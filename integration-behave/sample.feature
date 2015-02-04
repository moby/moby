Feature: showing what we can do with behave


Scenario: example testing inspect output
    Given an empty environment
     When I start container "test" with "busybox top"
      And I wait for the container "test" to be running
     Then the "State.Running" attribute of container "test" should be "true"
      And the "Path" attribute of container "test" should be "top"
      And the "Config.Image" attribute of container "test" should be "busybox"


Scenario: example testing ps output
    Given an empty environment
     When I start container "test" with "busybox top"
      And I wait for the container "test" to be running
     Then the IMAGE column of ps output for container "test" is "busybox:latest"
      And the COMMAND column of ps output for container "test" is ""top""


Scenario: example sending input to a container
    Given a container "test" started with "busybox"
     When I send "exit" to container "test"
     Then the container "test" stops


Scenario: example reading container's output
    Given an empty environment
     When I start container "test" with "busybox echo -n pouet"
     Then the container "test" outputs "pouet"

