
import argparse
import os
import time
from peewee import *
from playhouse.db_url import connect
import requests
from bs4 import BeautifulSoup
from pathlib import Path

db = connect('mysql://sticker:sticker@localhost:3306/sticker')
base_url = "https://tlgrm.eu"
url = "https://tlgrm.eu/stickers?page="
download_dir = str(os.path.join(Path.home(), "Downloads/tg-stickers/"))
headers = {'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/39.0.2171.95 Safari/537.36'}


class BaseModel(Model):

    def __repr__(self):
        name = self.__class__.__name__
        properties = ('{}=({})'.format(k, v) for k, v in self.__dict__.items())
        s = '\n<{} \n  {}>'.format(name, '\n '.join(properties))
        return s


class Sticker(BaseModel):
    sticker_name = CharField()
    album_id = CharField()
    album_name = CharField()
    url = TextField(null=True)

    class Meta:
        database = db
        primary_key = CompositeKey('sticker_name', 'album_id')


def download_sticker(url, filename):
    local_filename = download_dir + filename
    if os.path.exists(local_filename) and os.path.getsize(local_filename) > 0:
        return local_filename

    dir = local_filename[:local_filename.rindex('/')]
    if not os.path.exists(dir):
        os.makedirs(dir)
        
    print('downloading {} to {}'.format(url, local_filename))
    with requests.get(url, stream=True) as r:
        r.raise_for_status()
        with open(local_filename, 'wb') as f:
            for chunk in r.iter_content(chunk_size=8192):
                f.write(chunk)
    return local_filename


def parse_album(url):
    print('parsing {}'.format(url))
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
            postfix = '.webp'
        sticker_url = base_sticker_url + postfix
        print('parsing sticker_url {}'.format(sticker_url))

        try:
            local_filename = download_sticker(sticker_url, filename + postfix)
        except requests.exceptions.HTTPError as e:
            local_filename = download_dir + filename
            if os.path.exists(local_filename):
                os.remove(local_filename)

            if e.response.status_code == 404:
                if is_lottie:
                    is_lottie = False
                    postfix = '.webp'
                    sticker_url = base_sticker_url + postfix
                    print('parsing sticker_url {}'.format(sticker_url))
                    try:
                        local_filename = download_sticker(sticker_url, filename + postfix)
                    except Exception as e:
                        local_filename = download_dir + filename
                        if os.path.exists(local_filename):
                            os.remove(local_filename)

                        if e.response.status_code == 404:
                            print('no sticker {}'.format(sticker_url))
                            break
                else:
                    print('no sticker {}'.format(sticker_url))
                    break
            else:
                print('no sticker {}'.format(sticker_url))
                break
        
        sticker_name = str(i) + postfix
        try:
            sticker = Sticker.get(Sticker.sticker_name == sticker_name, Sticker.album_id == album_id)
        except Sticker.DoesNotExist:
            print('sticker {} does not exists'.format(album_id + '/' + sticker_name))
            sticker = Sticker.create(sticker_name=sticker_name, album_id=album_id, album_name=album_name, url=local_filename)
            sticker.save()
            continue

        if sticker:
            print('sticker {} already exists'.format(album_id + '/' + sticker_name))
            continue

        sticker = Sticker.create(sticker_name=sticker_name, album_id=album_id, album_name=album_name, url=local_filename)
        sticker.save()


def main(album):
    db.connect()
    db.create_tables([Sticker])

    if album:
        parse_album(base_url + '/stickers/' + album)
    else:    
        for i in range(1, 1000):
            page_url = url + str(i)
            r = requests.get(page_url, headers=headers)
            print('spidering {}'.format(page_url))
            soup = BeautifulSoup(r.text, 'html.parser')
            albums = soup.select('.stickerbox')
            print('albums found {}'.format(len(albums)))
            if len(albums) == 0:
                print('no new page')
                break

            for a in albums:
                parse_album(a['href'])
                time.sleep(10)

    db.close()


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--album', type=str)
    args = parser.parse_args()
    main(args.album)