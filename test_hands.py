import os
from dotenv import load_dotenv
from openai import OpenAI
import zhipuai

load_dotenv()

print('🔧 TESTING PAID KEYS...')
print('-'*30)

# TEST 1: KIMI (Moonshot)
print('1. Testing Kimi (Moonshot)...')
try:
 client = OpenAI(api_key=os.getenv('KIMI_API_KEY'), base_url='https://api.moonshot.cn/v1')
 resp = client.chat.completions.create(model='moonshot-v1-8k', messages=[{'role':'user','content':'Say KIMI WORKS'}])
 print('✅ KIMI:', resp.choices[0].message.content)
except Exception as e:
 print('❌ KIMI FAIL:', e)

# TEST 2: GLM (Zhipu)
print('
2. Testing GLM (Zhipu)...')
try:
 zhipuai.api_key = os.getenv('GLM_API_KEY')
 response = zhipuai.model_api.invoke(model='glm-4', prompt=[{'role':'user','content':'Say GLM WORKS'}])
 print('✅ GLM:', response['choices'][0]['content'])
except Exception as e:
 print('❌ GLM FAIL:', e)

print('-'*30)
