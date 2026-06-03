import base64
from Crypto.Cipher import AES
from Crypto.Util.Padding import unpad

def decrypt_cbc(data_b64, key_str="yx$5LT9yJz9MQ(O7"):
    """CBC 模式解密（对应 aesDecrypt）"""
    key = key_str.encode('utf-8')
    iv = key  # IV 等于 key
    
    ciphertext = base64.b64decode(data_b64)
    cipher = AES.new(key, AES.MODE_CBC, iv)
    decrypted = unpad(cipher.decrypt(ciphertext), AES.block_size)
    return decrypted.decode('utf-8')

def decrypt_ecb(data_b64, key_str="yx$5LT9yJz9MQ(O7"):
    """ECB 模式解密（对应 aesEncrypt 的逆过程）"""
    # 需要先生成和 aesEncrypt 一样的密钥
    r = []
    o = 0
    for c in range(16):
        r.append(key_str[o % len(key_str)])
        o = (o + 7) % len(key_str)
    key = ''.join(r).encode('utf-8')
    
    ciphertext = base64.b64decode(data_b64)
    cipher = AES.new(key, AES.MODE_ECB)
    decrypted = unpad(cipher.decrypt(ciphertext), AES.block_size)
    return decrypted.decode('utf-8')

# 尝试用 ECB 解密失败的几段
failed_data = [
    "ceMURmlsABoCBg81pXr3dgRS0GXm/pcxsMCxaZMOONu8wc0hnlp5WqKLNUkSVLt3",
    "ceMURmlsABoCBg81pXr3durHVIRXYc3oY0xmu+sc6wYpysw6qNhvyjDLmsx+xmuaignlgYZi0Pe8MNaVUQqorQxbpKbp6IA0FVLpDUwxnQ8BSuCrmGoNEXaX1tbf3cxBlw8dEaxwTvasSa43Fyg8k1zJpuK1w45l2z1grbjn3/LU89j+u8l5Dgyavx2Lxr9aoHcIoae1IOnUgh+g1YfRDf0yEAwM1HHPTX1w7x/hYm3rTfHjaa3wL2LVYEGLA7Io"
]

for data in failed_data:
    try:
        result = decrypt_ecb(data)
        print(f"ECB 解密成功: {result}")
    except Exception as e:
        print(f"ECB 解密失败: {e}")
    
    try:
        result = decrypt_cbc(data)
        print(f"CBC 解密成功: {result}")
    except Exception as e:
        print(f"CBC 解密失败: {e}")
