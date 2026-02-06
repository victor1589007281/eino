"""Memory SDK Setup"""

from setuptools import setup, find_packages

with open("README.md", "r", encoding="utf-8") as fh:
    long_description = fh.read()

setup(
    name="memory-sdk",
    version="1.0.0",
    author="VDocsTool Team",
    author_email="",
    description="Python SDK for Memory Service",
    long_description=long_description,
    long_description_content_type="text/markdown",
    url="https://github.com/cloudwego/eino/vdocstool",
    packages=find_packages(),
    classifiers=[
        "Development Status :: 4 - Beta",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: Apache Software License",
        "Operating System :: OS Independent",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "Programming Language :: Python :: 3.12",
    ],
    python_requires=">=3.8",
    install_requires=[
        "requests>=2.25.0",
    ],
    extras_require={
        "grpc": [
            "grpcio>=1.50.0",
            "grpcio-tools>=1.50.0",
            "protobuf>=4.0.0",
        ],
        "dev": [
            "pytest>=7.0.0",
            "pytest-asyncio>=0.20.0",
            "pytest-cov>=4.0.0",
        ],
    },
)
