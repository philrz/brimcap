#!/bin/bash
exec /opt/zeek/bin/zeek -C -r - --exec "event zeek_init() { Log::disable_stream(PacketFilter::LOG); Log::disable_stream(LoadedScripts::LOG); Log::disable_stream(Telemetry::LOG); }" local
