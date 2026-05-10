from sqlalchemy import Column, Integer, String, DateTime, Boolean, Text, ForeignKey, Enum
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import relationship
from datetime import datetime
import enum

Base = declarative_base()


class UserRole(str, enum.Enum):
    """用户角色"""
    ADMIN = "admin"
    DEVELOPER = "developer"
    USER = "user"


class PluginStatus(str, enum.Enum):
    """插件状态"""
    PENDING = "pending"  # 待审核
    APPROVED = "approved"  # 已上架
    REJECTED = "rejected"  # 已拒绝
    ARCHIVED = "archived"  # 已下架


class LicenseType(str, enum.Enum):
    """授权类型"""
    ONE_TIME = "one_time"  # 一次性购买
    SUBSCRIPTION = "subscription"  # 订阅


class User(Base):
    """用户表"""
    __tablename__ = "users"

    id = Column(Integer, primary_key=True, index=True)
    username = Column(String(50), unique=True, index=True, nullable=False)
    email = Column(String(100), unique=True, index=True, nullable=False)
    hashed_password = Column(String(255), nullable=False)
    role = Column(Enum(UserRole), default=UserRole.USER)
    is_active = Column(Boolean, default=True)
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    # 关系
    plugins = relationship("Plugin", back_populates="author")
    orders = relationship("Order", back_populates="user")


class Plugin(Base):
    """插件表"""
    __tablename__ = "plugins"

    id = Column(Integer, primary_key=True, index=True)
    name = Column(String(100), nullable=False)
    slug = Column(String(100), unique=True, index=True, nullable=False)
    description = Column(Text)
    version = Column(String(20), nullable=False)
    author_id = Column(Integer, ForeignKey("users.id"), nullable=False)
    runtime = Column(String(20), nullable=False)  # python/nodejs
    trigger = Column(String(255), nullable=False)
    platforms = Column(Text)  # JSON array

    # 定价
    price_type = Column(Enum(LicenseType), default=LicenseType.ONE_TIME)
    price = Column(Integer, default=0)  # 价格（分）
    monthly_price = Column(Integer, default=0)  # 月付价格（分）
    yearly_price = Column(Integer, default=0)  # 年付价格（分）

    # 状态
    status = Column(Enum(PluginStatus), default=PluginStatus.PENDING)
    downloads = Column(Integer, default=0)
    rating = Column(Integer, default=0)

    # 文件
    file_path = Column(String(255))  # 插件文件路径
    file_size = Column(Integer)  # 文件大小（字节）

    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    # 关系
    author = relationship("User", back_populates="plugins")
    orders = relationship("Order", back_populates="plugin")
    licenses = relationship("License", back_populates="plugin")


class Order(Base):
    """订单表"""
    __tablename__ = "orders"

    id = Column(Integer, primary_key=True, index=True)
    order_no = Column(String(50), unique=True, index=True, nullable=False)
    user_id = Column(Integer, ForeignKey("users.id"), nullable=False)
    plugin_id = Column(Integer, ForeignKey("plugins.id"), nullable=False)

    # 订单信息
    amount = Column(Integer, nullable=False)  # 金额（分）
    license_type = Column(Enum(LicenseType), nullable=False)

    # 支付信息
    payment_method = Column(String(20))  # alipay/wechat/stripe
    payment_status = Column(String(20), default="pending")  # pending/paid/failed/refunded
    paid_at = Column(DateTime)

    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    # 关系
    user = relationship("User", back_populates="orders")
    plugin = relationship("Plugin", back_populates="orders")


class License(Base):
    """授权证书表"""
    __tablename__ = "licenses"

    id = Column(Integer, primary_key=True, index=True)
    license_key = Column(String(100), unique=True, index=True, nullable=False)
    plugin_id = Column(Integer, ForeignKey("plugins.id"), nullable=False)
    user_id = Column(Integer, ForeignKey("users.id"), nullable=False)
    device_id = Column(String(100), nullable=False)

    # 授权信息
    license_type = Column(Enum(LicenseType), nullable=False)
    expires_at = Column(DateTime)
    is_active = Column(Boolean, default=True)

    # 签名
    signature = Column(Text, nullable=False)

    created_at = Column(DateTime, default=datetime.utcnow)
    last_verified_at = Column(DateTime)

    # 关系
    plugin = relationship("Plugin", back_populates="licenses")
