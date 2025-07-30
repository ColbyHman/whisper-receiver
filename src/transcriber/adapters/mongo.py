from pymongo import MongoClient

def get_db_collection(client_url, database, collection):
    """Returns Database Client"""

    try:
        client = MongoClient(client_url)
    except Exception as e:
        raise Exception("Could not get MongoClient: ", e)

    try:
        db = client[database]
    except Exception as e:
        raise Exception("Could not get Database: ", e)

    try:
        return db[collection] 
    except Exception as e:
        raise Exception("Could not get Collection: ", e)

def insert(item, collection):

    try:
        collection.insert_one(item)
    except Exception as e:
        raise Exception("Could not insert item")