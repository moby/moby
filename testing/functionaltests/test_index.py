#!/usr/bin/python

import os
username, password = os.environ['DOCKER_CREDS'].split(':')

from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.common.keys import Keys
from selenium.webdriver.support.ui import Select
from selenium.common.exceptions import NoSuchElementException
import unittest, time, re

class Docker(unittest.TestCase):
    def setUp(self):
        self.driver = webdriver.PhantomJS()
        self.driver.implicitly_wait(30)
        self.base_url = "http://www.docker.io/"
        self.verificationErrors = []
        self.accept_next_alert = True

    def test_docker(self):
        driver = self.driver
        print "Login into {0} as login user {1} ...".format(self.base_url,username)
        driver.get(self.base_url + "/")
        driver.find_element_by_link_text("INDEX").click()
        driver.find_element_by_link_text("login").click()
        driver.find_element_by_id("id_username").send_keys(username)
        driver.find_element_by_id("id_password").send_keys(password)
        print "Checking login user ..."
        driver.find_element_by_css_selector("input[type=\"submit\"]").click()
        try: self.assertEqual("test", driver.find_element_by_css_selector("h3").text)
        except AssertionError as e: self.verificationErrors.append(str(e))
        print "Login user {0} found".format(username)

    def is_element_present(self, how, what):
        try: self.driver.find_element(by=how, value=what)
        except NoSuchElementException, e: return False
        return True

    def is_alert_present(self):
        try: self.driver.switch_to_alert()
        except NoAlertPresentException, e: return False
        return True

    def close_alert_and_get_its_text(self):
        try:
            alert = self.driver.switch_to_alert()
            alert_text = alert.text
            if self.accept_next_alert:
                alert.accept()
            else:
                alert.dismiss()
            return alert_text
        finally: self.accept_next_alert = True

    def tearDown(self):
        self.driver.quit()
        self.assertEqual([], self.verificationErrors)

if __name__ == "__main__":
    unittest.main()
