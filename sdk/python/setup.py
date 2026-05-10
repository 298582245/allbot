from setuptools import setup, find_packages

setup(
    name="allbot-sdk",
    version="0.1.0",
    description="AllBot Python SDK for plugin development",
    author="AllBot",
    packages=find_packages(),
    install_requires=[
        "grpcio>=1.50.0",
        "grpcio-tools>=1.50.0",
    ],
    python_requires=">=3.7",
    classifiers=[
        "Development Status :: 3 - Alpha",
        "Intended Audience :: Developers",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.7",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
    ],
)
