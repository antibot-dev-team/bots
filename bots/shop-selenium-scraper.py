#!/usr/bin/env python3

from selenium import webdriver

URL = "http://localhost"


class Bot:
    def __init__(self):
        """
        Init bot
        """
        options = webdriver.ChromeOptions()
        options.add_argument("--headless")
        self.driver = webdriver.Chrome(executable_path="../driver/chromedriver", options=options)

    def get_pages(self) -> list:
        """
        Get available shop pages
        :return: list of unique pages
        """
        pagination = self.driver.find_element_by_class_name("page-numbers").find_elements_by_tag_name("li")

        links = []
        for element in pagination[1:]:
            link = element.find_element_by_tag_name("a").get_property("href")
            links.append(link)

        return list(set(links))

    def get_page_products(self) -> list:
        """
        Get available products per page
        :return: list of unique products
        """
        products = self.driver.find_element_by_class_name("products").find_elements_by_tag_name("li")

        links = []
        for product in products:
            link = product.find_element_by_class_name("woocommerce-LoopProduct-link").get_property("href")
            links.append(link)

        return list(set(links))

    def scrape_product(self) -> tuple:
        """
        Crape particular product
        :return: title and price as tuple
        """
        title = self.driver.find_element_by_class_name("product_title").text

        try:
            price = self.driver.find_element_by_xpath(
                '/html/body/div[1]/div[2]/div/div[2]/main/div[2]/div[2]/p/span/bdi').text
        except:
            price = self.driver.find_element_by_xpath(
                '/html/body/div[1]/div[2]/div/div[2]/main/div[2]/div[2]/p/ins/span/bdi').text

        return title, price

    def scrape(self):
        """
        Scrape shop products for titles and prices
        :return: None
        """
        self.driver.get(URL)
        self.driver.find_element_by_xpath('/html/body/div/header/div[2]/div/nav/div[1]/ul/li[6]/a').click()

        back_url = self.driver.current_url

        products = self.get_page_products()

        pages = self.get_pages()
        for page in pages:
            self.driver.get(page)
            products.extend(self.get_page_products())

        for product in products:
            self.driver.get(product)
            title, price = self.scrape_product()
            print(title, price)
            self.driver.get(back_url)

    def __del__(self):
        """
        Close driver on exit
        :return: None
        """
        try:
            self.driver.close()
            self.driver.quit()
        except:
            pass


if __name__ == "__main__":
    bot = Bot()
    bot.scrape()
