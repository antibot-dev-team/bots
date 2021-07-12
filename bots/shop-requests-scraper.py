#!/usr/bin/env python3

from concurrent.futures import ThreadPoolExecutor, as_completed
from re import findall
from typing import Tuple

from requests import get

URL = "http://localhost"


def get_product_info(link: str) -> Tuple[str, str]:
    """
    Parse product info
    :param link: link to the product
    :return: tuple with title and price
    """
    resp_text = get(link).text
    title = findall(r'<h1 class="product_title entry-title">(.*?)</h1>', resp_text)
    price = findall(r'priceSpecification":{"price":"(.*?)"', resp_text)

    title = title[0] if title else "Unknown"
    price = price[0] if price else "Unknown"
    return title, price


if __name__ == "__main__":
    products = []
    for index in range(1, 10):
        resp = get(f"{URL}/shop/page/{index}/")
        if resp.status_code == 404:
            break
        resp_text = resp.text
        products_page = findall(r'<a href="(http://.*/product/.*/)"', resp_text)
        products.extend(products_page)

    with ThreadPoolExecutor(max_workers=10) as executor:
        futures = [executor.submit(get_product_info, product_link) for product_link in products]

    all_products = []
    for future in as_completed(futures):
        result = future.result()
        title = result[0]
        price = result[1]
        all_products.append((title, price))
        print(title, price)
