#!/usr/bin/env ruby
# frozen_string_literal: true
#
# Copyright (C) 2017 Harald Sitter <sitter@kde.org>
#
# This program is free software; you can redistribute it and/or
# modify it under the terms of the GNU General Public License as
# published by the Free Software Foundation; either version 2 of
# the License or (at your option) version 3 or any later version
# accepted by the membership of KDE e.V. (or its successor approved
# by the membership of KDE e.V.), which shall act as a proxy
# defined in Section 14 of version 3 of the license.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.

# This should only use core ruby as it is used on non-standardized systems.
require 'net/http'

# HTTP server side event.
# Body looks like this:
#   event:stdout
#   data:hi there
#
class SSEvent
  attr_reader :name # This is a free form string.
  attr_reader :data # Free form data.

  def initialize(body)
    matchdata = /event:(?<name>.+)\ndata:(?<data>.*)/n.match(body)
    unless matchdata
      warn 'Failed to parse SSEvent'
      warn body.inspect
      return
    end
    @name = matchdata[:name].strip
    @data = matchdata[:data].strip
  end

  # Does whatever needs doing for this event.
  # stdout/err -> puts
  # error -> exit
  def run
    case name
    when 'error'
      if data && !data.empty?
        STDERR.puts data
        exit 1
      end
      exit 0
    when 'stderr' then STDERR.puts(data)
    else STDOUT.puts(data)
    end
  end
end

uri = URI.parse(ARGV.pop)
Net::HTTP.start(uri.host, uri.port, use_ssl: uri.scheme == 'https') do |http|
  http.read_timeout = 60 * 60
  request = Net::HTTP::Get.new uri.request_uri
  http.request(request) do |response|
    response.read_body do |chunk|
      SSEvent.new(chunk).run
    end
  end
end
