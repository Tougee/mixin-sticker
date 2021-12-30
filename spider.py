
import argparse
import os
import time
from peewee import *
from playhouse.db_url import connect
import requests
from bs4 import BeautifulSoup
from pathlib import Path
import uuid
import logging

db = connect('mysql://sticker:sticker@localhost:3306/sticker')
logging.basicConfig(filename='spider.log', encoding='utf-8', level=logging.DEBUG, format='%(asctime)s %(message)s', datefmt='%m/%d/%Y %I:%M:%S %p')
parser = argparse.ArgumentParser()

base_url = "https://tlgrm.eu"
url = "https://tlgrm.eu/stickers?page="

tg_download_dir = str(os.path.join(Path.home(), "Downloads/mixin-sticker/tg-stickers/"))
lottiefiles_download_dir = str(os.path.join(Path.home(), "Downloads/mixin-sticker/lottiefiles/"))

headers = {'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/39.0.2171.95 Safari/537.36'}

tg_sticker_size = '192'

class BaseModel(Model):

    def __repr__(self):
        name = self.__class__.__name__
        properties = ('{}=({})'.format(k, v) for k, v in self.__dict__.items())
        s = '\n<{} \n  {}>'.format(name, '\n '.join(properties))
        return s


class Sticker(BaseModel):
    sticker_id = CharField(primary_key=True)
    url = CharField(unique=True, max_length=2048)
    sticker_name = CharField(null=True)
    album_id = CharField(null=True)
    album_name = CharField(null=True)
    local_url = CharField(null=True, max_length=2048)
    mixin_sticker_id = CharField(null=True)

    class Meta:
        database = db


def download_sticker(url, dir, filename, size=None):
    local_filename = dir + filename
    dir = local_filename[:local_filename.rindex('/')]
    if not os.path.exists(dir):
        os.makedirs(dir)
        
    if url.endswith('.json'):
        download_url = url
    else:
        split_index= url.rindex('/')
        if size:
            download_url = url[:split_index] + '/' + size + '/' + url[split_index + 1:]
        else:
            download_url = url[:split_index] + '/' + url[split_index + 1:]

    if os.path.exists(local_filename) and os.path.getsize(local_filename) > 0:
        return download_url, local_filename

    logging.debug('downloading {} to {}'.format(download_url, local_filename))
    with requests.get(download_url, stream=True) as r:
        r.raise_for_status()
        with open(local_filename, 'wb') as f:
            for chunk in r.iter_content(chunk_size=8192):
                f.write(chunk)

    return download_url, local_filename


def parse_album(url):
    logging.debug('parsing {}'.format(url))
    r = requests.get(url, headers=headers)
    soup = BeautifulSoup(r.text, 'html.parser')
    image_meta = soup.find_all('meta', property='og:image')
    content = str(image_meta[0]['content'])
    album_id = content[20: content.rindex('/')]
    album_name=url[url.rindex('/') + 1:]

    is_lottie = True
    for i in range(1, 5000):
        filename = album_id + '/' + str(i)
        base_sticker_url = base_url + content[:content.rindex('/')] + '/' + str(i)
        if is_lottie:
            postfix = '.json'
        else:
            postfix = '.png'
        sticker_url = base_sticker_url + postfix
        logging.debug('parsing sticker_url {}'.format(sticker_url))

        try:
            download_url, local_filename = download_sticker(sticker_url, tg_download_dir, filename + postfix, tg_sticker_size)
        except requests.exceptions.HTTPError as e:
            local_filename = tg_download_dir + filename
            if os.path.exists(local_filename):
                os.remove(local_filename)

            if e.response.status_code == 404:
                if is_lottie:
                    is_lottie = False
                    postfix = '.png'
                    sticker_url = base_sticker_url + postfix
                    logging.debug('parsing sticker_url {}'.format(sticker_url))
                    try:
                        download_url, local_filename = download_sticker(sticker_url, tg_download_dir, filename + postfix, tg_sticker_size)
                    except Exception as e:
                        local_filename = tg_download_dir + filename
                        if os.path.exists(local_filename):
                            os.remove(local_filename)

                        if e.response.status_code == 404:
                            logging.info('no sticker {}'.format(sticker_url))
                            break
                else:
                    logging.info('no sticker {}'.format(sticker_url))
                    break
            else:
                logging.info('no sticker {}'.format(sticker_url))
                break
        
        sticker_name = str(i) + postfix
        try:
            sticker = Sticker.get_or_none(Sticker.sticker_name == sticker_name, Sticker.album_id == album_id)
        except Sticker.DoesNotExist:
            logging.debug('sticker {} does not exists'.format(album_id + '/' + sticker_name))
            sticker = Sticker.create(sticker_id=str(uuid.uuid4()), url=download_url, sticker_name=sticker_name, album_id=album_id, album_name=album_name, local_url=local_filename)
            sticker.save()
            continue

        if sticker:
            logging.debug('sticker {} already exists'.format(album_id + '/' + sticker_name))
            continue

        sticker = Sticker.create(sticker_id=str(uuid.uuid4()), url=download_url, sticker_name=sticker_name, album_id=album_id, album_name=album_name, local_url=local_filename)
        sticker.save()


def parse_json_url(url):
    sticker = Sticker.get_or_none(Sticker.url == url)
    if sticker:
        logging.info('sticker already exists {}'.format(url))
        return

    file_name = url[url.rindex('/') + 1:]
    try:
        download_url, local_filename = download_sticker(url, lottiefiles_download_dir, file_name)
    except Exception as e:
        logging.info('no sticker ', e)
        return

    sticker = Sticker.create(sticker_id=str(uuid.uuid4()), url=download_url, sticker_name=file_name, local_url=local_filename)
    sticker.save()


def route_args(args):
    url = args.url

    if url and url.endswith('.json'):
        parse_json_url(url)
    elif args.album:
        parse_album(base_url + '/stickers/' + args.album)
    elif args.tg:    
        for i in range(1, 1000):
            page_url = url + str(i)
            r = requests.get(page_url, headers=headers)
            logging.debug('spidering {}'.format(page_url))
            soup = BeautifulSoup(r.text, 'html.parser')
            albums = soup.select('.stickerbox')
            logging.debug('albums found {}'.format(len(albums)))
            if len(albums) == 0:
                logging.info('no new page')
                break

            for a in albums:
                parse_album(a['href'])
                time.sleep(10)
    else:
        parser.print_help()


def main():
    db.connect()
    db.create_tables([Sticker])

    parser.add_argument('--url', type=str, help='lottie json url, e.g., https://assets9.lottiefiles.com/packages/lf20_muiaursk.json')
    parser.add_argument('--album', type=str, help='Telegram sticker album name, e.g., stpcts')
    parser.add_argument('--tg', type=bool, default=False, help='Spider Telegram stickers from https://tlgrm.eu')
    args = parser.parse_args()

    route_args(args)

    db.close()


if __name__ == '__main__':
    main()