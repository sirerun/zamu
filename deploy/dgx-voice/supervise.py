import asyncio, os, pathlib, shutil, signal, subprocess, sys, time, urllib.request
os.environ.update(PYTHONPATH='/prepared/pocket:/prepared/deps:/deps', HF_HUB_OFFLINE='1', TRANSFORMERS_OFFLINE='1', OMP_NUM_THREADS='2', OPENBLAS_NUM_THREADS='2', MISTT_CONVERSATION_DEVICE='cpu', MISTT_CONVERSATION_THREADS='2')
pathlib.Path('/run/voice').mkdir(mode=0o700,exist_ok=True)
for name in ('mistt','narrate'):
 p=pathlib.Path('/run/voice')/name
 p.write_bytes((pathlib.Path('/credentials')/name/'token').read_bytes());p.chmod(0o600)
children=[]
def start(args,cwd=None):
 p=subprocess.Popen(args,cwd=cwd);children.append(p);return p
def stop(*_):
 for p in children:
  if p.poll() is None:p.terminate()
 for p in children:
  try:p.wait(timeout=20)
  except subprocess.TimeoutExpired:p.kill()
 if _:sys.exit(0)
signal.signal(signal.SIGTERM,stop);signal.signal(signal.SIGINT,stop)
start([sys.executable,'/source/mistt/serve.py','--host','127.0.0.1','--port','8793','--token-file','/run/voice/mistt','--model-directory','/prepared/whisper'],cwd='/source/mistt')
start([sys.executable,'/source/narrate/server.py','--host','0.0.0.0','--port','8092','--token-file','/run/voice/narrate','--model-config','/models/config.yaml','--model-config-sha256','193c319c5148f03fc8310423dbaa9022de557dce97ed89b06d668f1af911c859','--anna-asset','/models/anna.safetensors','--anna-sha256','5ea82f78db006c9fd34e32ddd5aae82674b5b32646097977436458d00af80dfa'])
for port,name in ((8793,'mistt'),(8092,'narrate')):
 for i in range(120):
  if any(p.poll() is not None for p in children):raise RuntimeError('speech process exited')
  try:
   req=urllib.request.Request('http://127.0.0.1:%d/healthz'%port,headers={'Authorization':'Bearer '+(pathlib.Path('/run/voice')/name).read_text().strip()})
   with urllib.request.urlopen(req,timeout=2) as r:
    if r.status==200:break
  except Exception:time.sleep(1)
 else:raise RuntimeError('speech startup health deadline')
start(['/source/mistt-conversation','--listen','127.0.0.1:8792','--recognizer','http://127.0.0.1:8793/v1/recognize','--token-file','/run/voice/mistt','--recognizer-token-file','/run/voice/mistt','--model','openai/whisper-base.en@911407f4214e0e1d82085af863093ec0b66f9cd6'])
async def proxy(reader,writer):
 peer=None
 try:
  r,w=await asyncio.open_connection('127.0.0.1',8792);peer=w
  async def copy(src,dst):
   while data:=await src.read(65536):dst.write(data);await dst.drain()
  tasks=[asyncio.create_task(copy(reader,w)),asyncio.create_task(copy(r,writer))]
  await asyncio.wait(tasks,return_when=asyncio.FIRST_COMPLETED)
  for t in tasks:t.cancel()
  await asyncio.gather(*tasks,return_exceptions=True)
 finally:
  writer.close()
  if peer:peer.close()
async def main():
 server=await asyncio.start_server(proxy,'0.0.0.0',8794)
 print('VOICE_SERVICES_READY',flush=True)
 async with server:
  while all(p.poll() is None for p in children):await asyncio.sleep(1)
 raise RuntimeError('speech process exited')
try:asyncio.run(main())
finally:stop()
