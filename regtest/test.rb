# -*- encoding: utf-8 -*-
require 'httpclient'
require 'pp'

def logs(response)
  h = response.http_header
  request = "#{h.request_method} #{h.request_uri.request_uri} HTTP/#{h.http_version}"

  puts request
  puts
  puts response.body
  puts
  puts

  return request
end

def bench()
  start = Time.now
  request = logs(yield)
  stop = Time.now

  time = stop - start
  $total_time += time

  STDERR.puts request
  STDERR.puts "#{time}[s]"
  STDERR.puts
end

$total_time = 0.0

isu17a = 'http://203.104.208.219'

client1 = HTTPClient.new
bench { client1.get(isu17a + '/initialize') }
bench { client1.get(isu17a + '/') }
bench { client1.get(isu17a + '/login') }
bench { client1.post(isu17a + '/signup', password: 'tonyny31', grade: 'micro', email: 'tony1@moris.io') }
bench { client1.post(isu17a + '/login', password: 'tonyny31', email: 'tony1@moris.io') }
bench { client1.get(isu17a + '/') }
bench { client1.get(isu17a + '/data') }
bench { client1.get(isu17a + '/modify') }
bench { client1.post(isu17a + '/modify', service: 'ken', keys: '5460000') }
bench { client1.post(isu17a + '/modify', param_value: '8792435', service: 'ken2', param_name: 'zipcode') }

client2 = HTTPClient.new

STDERR.puts "TOTAL: #{$total_time}[s]"

# MEMO: cookieの値のチェック
# client1.cookies[0].value
