from redis import Redis

def get_redis_client(host: str = "localhost", port: int = 6379, db: int = 0) -> Redis:
    """Returns Redis Client"""

    try:
        client = Redis(host=host, port=port, db=db)
        client.ping()
        return client
    except Exception as e:
        raise Exception("Could not get Redis Client: ", e)