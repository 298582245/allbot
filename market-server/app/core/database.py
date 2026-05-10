from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
import os

# 数据库配置
DATABASE_URL = os.getenv("DATABASE_URL", "sqlite:///./market.db")

engine = create_engine(
    DATABASE_URL,
    connect_args={"check_same_thread": False} if "sqlite" in DATABASE_URL else {}
)

SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)


def get_db():
    """获取数据库会话"""
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()


def init_db():
    """初始化数据库"""
    from app.models.models import Base
    Base.metadata.create_all(bind=engine)
