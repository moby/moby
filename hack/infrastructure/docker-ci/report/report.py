#!/usr/bin/python

'''CONFIG_JSON is a json encoded string base64 environment variable. It is used
to clone docker-ci database, generate docker-ci report and submit it by email.
CONFIG_JSON data comes from the file /report/credentials.json inserted in this
container by deployment.py:

{ "DOCKER_CI_PUB":       "$(cat docker-ci_ssh_public_key.pub)",
  "DOCKER_CI_KEY":       "$(cat docker-ci_ssh_private_key.key)",
  "DOCKER_CI_ADDRESS":   "user@docker-ci_fqdn_server",
  "SMTP_USER":           "SMTP_server_user",
  "SMTP_PWD":            "SMTP_server_password",
  "EMAIL_SENDER":        "Buildbot_mailing_sender",
  "EMAIL_RCP":           "Buildbot_mailing_receipient" }  '''

import os, re, json, sqlite3, datetime, base64
import smtplib
from datetime import timedelta
from subprocess import call
from os import environ as env

TODAY = datetime.date.today()

# Load credentials to the environment
env['CONFIG_JSON'] = base64.b64decode(open('/report/credentials.json').read())

# Remove SSH private key as it needs more processing
CONFIG = json.loads(re.sub(r'("DOCKER_CI_KEY".+?"(.+?)",)','',
    env['CONFIG_JSON'], flags=re.DOTALL))

# Populate environment variables
for key in CONFIG:
    env[key] = CONFIG[key]

# Load SSH private key
env['DOCKER_CI_KEY'] = re.sub('^.+"DOCKER_CI_KEY".+?"(.+?)".+','\\1',
    env['CONFIG_JSON'],flags=re.DOTALL)

# Prevent rsync to validate host on first connection to docker-ci
os.makedirs('/root/.ssh')
open('/root/.ssh/id_rsa','w').write(env['DOCKER_CI_KEY'])
os.chmod('/root/.ssh/id_rsa',0600)
open('/root/.ssh/config','w').write('StrictHostKeyChecking no\n')


# Sync buildbot database from docker-ci
call('rsync {}:/data/buildbot/master/state.sqlite .'.format(
    env['DOCKER_CI_ADDRESS']), shell=True)

class SQL:
    def __init__(self, database_name):
        sql = sqlite3.connect(database_name)
        # Use column names as keys for fetchall rows
        sql.row_factory = sqlite3.Row
        sql = sql.cursor()
        self.sql = sql

    def query(self,query_statement):
        return self.sql.execute(query_statement).fetchall()

sql = SQL("state.sqlite")


class Report():

    def __init__(self,period='',date=''):
        self.data = []
        self.period = 'date' if not period else period
        self.date = str(TODAY) if not date else date
        self.compute()

    def compute(self):
        '''Compute report'''
        if self.period == 'week':
            self.week_report(self.date)
        else:
            self.date_report(self.date)


    def date_report(self,date):
        '''Create a date test report'''
        builds = []
        # Get a queryset with all builds from date
        rows = sql.query('SELECT * FROM builds JOIN buildrequests'
            ' WHERE builds.brid=buildrequests.id and'
            ' date(start_time, "unixepoch", "localtime") = "{0}"'
            ' GROUP BY number'.format(date))
        build_names = sorted(set([row['buildername'] for row in rows]))
        # Create a report build line for a given build
        for build_name in build_names:
            tried = len([row['buildername']
                for row in rows if row['buildername'] == build_name])
            fail_tests = [row['buildername'] for row in rows if (
                row['buildername'] == build_name and row['results'] != 0)]
            fail = len(fail_tests)
            fail_details = ''
            fail_pct = int(100.0*fail/tried) if  tried != 0 else 100
            builds.append({'name': build_name, 'tried': tried, 'fail': fail,
                'fail_pct': fail_pct, 'fail_details':fail_details})
        if builds:
            self.data.append({'date': date, 'builds': builds})


    def week_report(self,date):
        '''Add the week's date test reports to report.data'''
        date = datetime.datetime.strptime(date,'%Y-%m-%d').date()
        last_monday = date - datetime.timedelta(days=date.weekday())
        week_dates = [last_monday + timedelta(days=x) for x in range(7,-1,-1)]
        for date in week_dates:
            self.date_report(str(date))

    def render_text(self):
        '''Return rendered report in text format'''
        retval = ''
        fail_tests = {}
        for builds in self.data:
            retval += 'Test date: {0}\n'.format(builds['date'],retval)
            table = ''
            for build in builds['builds']:
                table += ('Build {name:15}   Tried: {tried:4}   '
                    ' Failures: {fail:4} ({fail_pct}%)\n'.format(**build))
                if build['name'] in fail_tests:
                    fail_tests[build['name']] += build['fail_details']
                else:
                    fail_tests[build['name']] = build['fail_details']
            retval += '{0}\n'.format(table)
            retval += '\n    Builds failing'
            for fail_name in fail_tests:
                retval += '\n' + fail_name + '\n'
                for (fail_id,fail_url,rn_tests,nr_errors,log_errors,
                 tracelog_errors) in fail_tests[fail_name]:
                    retval += fail_url + '\n'
            retval += '\n\n'
        return retval


# Send email
smtp_from = env['EMAIL_SENDER']
subject = '[docker-ci] Daily report for {}'.format(str(TODAY))
msg = "From: {}\r\nTo: {}\r\nSubject: {}\r\n\r\n".format(
    smtp_from, env['EMAIL_RCP'], subject)
msg = msg + Report('week').render_text()
server = smtplib.SMTP_SSL('smtp.mailgun.org')
server.login(env['SMTP_USER'], env['SMTP_PWD'])
server.sendmail(smtp_from, env['EMAIL_RCP'], msg)
