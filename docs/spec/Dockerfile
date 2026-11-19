# syntax=docker/dockerfile:1

# Toolchain for building, checking and formatting the behavior specs.

ARG PYTHON_VERSION=3.13

FROM python:${PYTHON_VERSION}-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends git graphviz \
    && rm -rf /var/lib/apt/lists/*
COPY requirements.txt /tmp/requirements.txt
RUN pip install --no-cache-dir -r /tmp/requirements.txt
WORKDIR /src
