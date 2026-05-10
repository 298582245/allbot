from fastapi import APIRouter, Depends, HTTPException, UploadFile, File
from sqlalchemy.orm import Session
from typing import List
import os
import shutil
from datetime import datetime

from app.core.database import get_db
from app.models.models import Plugin, PluginStatus, User
from app.main import get_current_active_user

router = APIRouter(prefix="/api/plugins", tags=["plugins"])

UPLOAD_DIR = os.getenv("UPLOAD_DIR", "./uploads")
os.makedirs(UPLOAD_DIR, exist_ok=True)


@router.get("/")
async def list_plugins(
    skip: int = 0,
    limit: int = 20,
    status: str = "approved",
    db: Session = Depends(get_db)
):
    """获取插件列表"""
    query = db.query(Plugin)

    if status:
        query = query.filter(Plugin.status == status)

    plugins = query.offset(skip).limit(limit).all()

    return {
        "total": query.count(),
        "items": [
            {
                "id": p.id,
                "name": p.name,
                "slug": p.slug,
                "description": p.description,
                "version": p.version,
                "runtime": p.runtime,
                "price": p.price,
                "downloads": p.downloads,
                "rating": p.rating,
                "author": p.author.username,
                "created_at": p.created_at.isoformat(),
            }
            for p in plugins
        ]
    }


@router.get("/{plugin_id}")
async def get_plugin(plugin_id: int, db: Session = Depends(get_db)):
    """获取插件详情"""
    plugin = db.query(Plugin).filter(Plugin.id == plugin_id).first()
    if not plugin:
        raise HTTPException(status_code=404, detail="Plugin not found")

    return {
        "id": plugin.id,
        "name": plugin.name,
        "slug": plugin.slug,
        "description": plugin.description,
        "version": plugin.version,
        "runtime": plugin.runtime,
        "trigger": plugin.trigger,
        "platforms": plugin.platforms,
        "price_type": plugin.price_type,
        "price": plugin.price,
        "monthly_price": plugin.monthly_price,
        "yearly_price": plugin.yearly_price,
        "status": plugin.status,
        "downloads": plugin.downloads,
        "rating": plugin.rating,
        "file_size": plugin.file_size,
        "author": {
            "id": plugin.author.id,
            "username": plugin.author.username,
        },
        "created_at": plugin.created_at.isoformat(),
        "updated_at": plugin.updated_at.isoformat(),
    }


@router.post("/")
async def create_plugin(
    name: str,
    description: str,
    version: str,
    runtime: str,
    trigger: str,
    file: UploadFile = File(...),
    current_user: User = Depends(get_current_active_user),
    db: Session = Depends(get_db)
):
    """上传插件"""
    # 生成 slug
    slug = name.lower().replace(" ", "-")

    # 检查 slug 是否已存在
    existing = db.query(Plugin).filter(Plugin.slug == slug).first()
    if existing:
        raise HTTPException(status_code=400, detail="Plugin with this name already exists")

    # 保存文件
    file_path = os.path.join(UPLOAD_DIR, f"{slug}-{version}.allbot")
    with open(file_path, "wb") as buffer:
        shutil.copyfileobj(file.file, buffer)

    file_size = os.path.getsize(file_path)

    # 创建插件记录
    plugin = Plugin(
        name=name,
        slug=slug,
        description=description,
        version=version,
        author_id=current_user.id,
        runtime=runtime,
        trigger=trigger,
        file_path=file_path,
        file_size=file_size,
        status=PluginStatus.PENDING,
    )

    db.add(plugin)
    db.commit()
    db.refresh(plugin)

    return {
        "id": plugin.id,
        "name": plugin.name,
        "slug": plugin.slug,
        "status": plugin.status,
        "message": "Plugin uploaded successfully. Waiting for approval."
    }


@router.get("/{plugin_id}/download")
async def download_plugin(
    plugin_id: int,
    current_user: User = Depends(get_current_active_user),
    db: Session = Depends(get_db)
):
    """下载插件"""
    plugin = db.query(Plugin).filter(Plugin.id == plugin_id).first()
    if not plugin:
        raise HTTPException(status_code=404, detail="Plugin not found")

    # TODO: 检查用户是否已购买

    # 增加下载次数
    plugin.downloads += 1
    db.commit()

    from fastapi.responses import FileResponse
    return FileResponse(
        plugin.file_path,
        media_type="application/octet-stream",
        filename=f"{plugin.slug}-{plugin.version}.allbot"
    )


@router.post("/{plugin_id}/purchase")
async def purchase_plugin(
    plugin_id: int,
    license_type: str,
    current_user: User = Depends(get_current_active_user),
    db: Session = Depends(get_db)
):
    """购买插件"""
    plugin = db.query(Plugin).filter(Plugin.id == plugin_id).first()
    if not plugin:
        raise HTTPException(status_code=404, detail="Plugin not found")

    # TODO: 创建订单和支付流程

    return {
        "message": "Purchase initiated",
        "plugin_id": plugin_id,
        "license_type": license_type
    }


@router.post("/verify")
async def verify_license(license_key: str, device_id: str, db: Session = Depends(get_db)):
    """验证授权"""
    from app.models.models import License

    license = db.query(License).filter(
        License.license_key == license_key,
        License.device_id == device_id,
        License.is_active == True
    ).first()

    if not license:
        raise HTTPException(status_code=404, detail="License not found or invalid")

    # 检查是否过期
    if license.expires_at and license.expires_at < datetime.utcnow():
        raise HTTPException(status_code=403, detail="License expired")

    # 更新最后验证时间
    license.last_verified_at = datetime.utcnow()
    db.commit()

    return {
        "valid": True,
        "plugin_id": license.plugin_id,
        "license_type": license.license_type,
        "expires_at": license.expires_at.isoformat() if license.expires_at else None
    }
