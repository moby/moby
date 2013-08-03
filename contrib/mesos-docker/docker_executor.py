#!/usr/bin/env python
import sys

import mesos
import mesos_pb2
import os
import signal
import json
import subprocess
from circus.process import Process


class DockerExecutor(mesos.Executor):
    def __init__(self):
        self.tasks = {}

    def kill_child_processes(self, parent_pid, sig=signal.SIGTERM):
        ps_command = subprocess.Popen("ps -o pid --ppid %d --noheaders" % parent_pid, shell=True, stdout=subprocess.PIPE)
        ps_output = ps_command.stdout.read()
        retcode = ps_command.wait()
        if retcode is 0:
            for pid_str in ps_output.split("\n")[:-1]:
                self.kill_child_processes(int(pid_str))
                try:
                    print "Killing %s" % pid_str
                    os.kill(int(pid_str), sig)
                    return True
                except:
                    print "Cannot kill %s" % pid_str

    def launchTask(self, driver, task):
        job = json.loads(task.data)
        print "Running task_id %s: %s" % (task.task_id.value, job['name'])
        frameworkDir = os.path.abspath(os.path.dirname(sys.argv[0]))
        execPath = os.path.join(frameworkDir, job['command'])
        job['env']['APP_PORT'] = str(job['port'])
        try:
            self.tasks[task.task_id.value] = Process(task.task_id, execPath, args=job['args'], env=job['env'])
        except Exception, e:
            print "Failing running task, exception: %s" % e
            update = mesos_pb2.TaskStatus()
            update.task_id.value = task.task_id.value
            update.state = mesos_pb2.TASK_FAILED
            update.data = 'data with a \0 byte'
            driver.sendStatusUpdate(update)
            return
        print "New task status: %s" % self.tasks[task.task_id.value].status
        update = mesos_pb2.TaskStatus()
        update.task_id.value = task.task_id.value
        update.state = mesos_pb2.TASK_RUNNING
        update.data = 'data with a \0 byte'
        driver.sendStatusUpdate(update)

    def killTask(self, driver, task_id):
        print "Asked to kill task %s..." % task_id.value
        try:
            task = self.tasks[task_id.value]
            self.kill_child_processes(task.pid)
            task.stop()
        except Exception, e:
            print "Asked to kill what I cannot: %s" % task_id.value
            print "Exception: %s" % e
            update = mesos_pb2.TaskStatus()
            update.task_id.value = task_id.value
            update.state = mesos_pb2.TASK_KILLED
            update.data = 'data with a \0 byte'
            driver.sendStatusUpdate(update)
            print "Sent status update"

    def frameworkMessage(self, driver, message):
        # Send it back to the scheduler.
        driver.sendFrameworkMessage(message)

if __name__ == "__main__":
    print "Starting executor"
    the_executor = DockerExecutor()
    driver = mesos.MesosExecutorDriver(the_executor)
    print "driver started"
    sys.exit(0 if driver.run() == mesos_pb2.DRIVER_STOPPED else 1)
