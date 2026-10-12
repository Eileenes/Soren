import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MobileApp } from '../mobile/MobileApp';
const mocks = vi.hoisted(() => ({ request: vi.fn() }));
vi.mock('../mobile/client', () => ({
 native: false, parsePair: vi.fn(),
 MobileClient: { restore: async () => ({request: mocks.request}), forget: vi.fn() },
}));
const bots = [{id:'soren', name:'Soren',avatar:'soren'}, {id:'other',name:'Other',avatar:'robo'}];
async function flush() { await act(async()=>{ for(let i=0;i<15;i++) await Promise.resolve(); }); }
afterEach(()=>{vi.useRealTimers();mocks.request.mockReset();});
describe('Mobile recovery',()=>{
 it('clears a transient connection failure after the next successful refresh', async()=>{
  vi.useFakeTimers();
  mocks.request.mockRejectedValueOnce(new Error('temporary offline'))
    .mockImplementation(async (_method,path)=>path==='/api/bots'?bots:[]);
  render(<MobileApp/>); await flush();
  expect(screen.getByRole('alert')).toHaveTextContent('temporary offline');
  await act(async()=>{await vi.advanceTimersByTimeAsync(3000);});
  expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  expect(screen.getByText('Mac 已连接')).toBeInTheDocument();
 });
 it('prevents bot switching during submission and keeps write errors after a refresh', async()=>{
  vi.useFakeTimers();
  let reject!: (e:Error)=>void;
  mocks.request.mockImplementation((method,path)=>method==='POST'?new Promise((_resolve,r)=>{reject=r}):Promise.resolve(path==='/api/bots'?bots:[]));
  render(<MobileApp/>); await flush();
  fireEvent.change(screen.getByRole('textbox',{name:'交给 Bot 的任务'}),{target:{value:'task'}});
  fireEvent.click(screen.getByRole('button',{name:'交给 Soren'})); await flush();
  expect(screen.getByRole('button',{name:'Other'})).toBeDisabled();
  await act(async()=>{reject(new Error('operation may have been submitted'));});
  await act(async()=>{await vi.advanceTimersByTimeAsync(3000);});
  expect(screen.getByRole('alert')).toHaveTextContent('operation may have been submitted');
  expect(screen.getByRole('button',{name:'Other'})).not.toBeDisabled();
 });
});
